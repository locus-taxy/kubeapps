import * as React from "react";

import { RouterAction } from "connected-react-router";
import { JSONSchema4 } from "json-schema";
import { IAppRepository, IChartState, IChartVersion, IRelease } from "../../shared/types";
import { ErrorSelector } from "../ErrorAlert";
import LoadingWrapper from "../LoadingWrapper";
import SelectRepoForm from "../SelectRepoForm";
import UpgradeForm from "../UpgradeForm";

interface IAppUpgradeProps {
  app: IRelease;
  appsIsFetching: boolean;
  appsError: Error | undefined;
  namespace: string;
  releaseName: string;
  repoName: string;
  selected: IChartState["selected"];
  deployed: IChartState["deployed"];
  upgradeApp: (
    version: IChartVersion,
    releaseName: string,
    namespace: string,
    values?: string,
    schema?: JSONSchema4,
  ) => Promise<boolean>;
  fetchChartVersions: (id: string) => Promise<IChartVersion[]>;
  getAppWithUpdateInfo: (releaseName: string, namespace: string) => void;
  getChartVersion: (id: string, chartVersion: string) => void;
  getDeployedChartVersion: (id: string, chartVersion: string) => void;
  push: (location: string) => RouterAction;
  goBack: () => RouterAction;
  // repo selector properties
  reposIsFetching: boolean;
  kubeappsNamespace: string;
  repoError?: Error;
  chartsError: Error | undefined;
  repo: IAppRepository;
  repos: IAppRepository[];
  checkChart: (repo: string, chartName: string) => any;
  fetchRepositories: () => void;
}

class AppUpgrade extends React.Component<IAppUpgradeProps> {
  public componentDidMount() {
    const { releaseName, getAppWithUpdateInfo, namespace } = this.props;
    getAppWithUpdateInfo(releaseName, namespace);
  }

  public componentDidUpdate(prevProps: IAppUpgradeProps) {
    const { app, repoName } = this.props;
    if (app && repoName) {
      const { chart } = app;
      if (
        chart &&
        chart.metadata &&
        chart.metadata.name &&
        chart.metadata.version &&
        (prevProps.app !== app || prevProps.repoName !== repoName)
      ) {
        const chartID = `${repoName}/${chart.metadata.name}`;
        this.props.getDeployedChartVersion(chartID, chart.metadata.version);
      }
    }
  }

  public render() {
    const {
      app,
      namespace,
      appsError,
      releaseName,
      appsIsFetching,
      repoName,
      selected,
      deployed,
      upgradeApp,
      push,
      goBack,
      fetchChartVersions,
      getChartVersion,
    } = this.props;
    if (appsError) {
      return (
        <ErrorSelector
          error={appsError}
          namespace={namespace}
          action="update"
          resource={releaseName}
        />
      );
    }
    if (appsIsFetching || !app || !app.updateInfo) {
      return <LoadingWrapper />;
    }
    const repo = repoName || app.updateInfo.repository.name;
    if (app && app.chart && app.chart.metadata && repo) {
      return (
        <div>
          <UpgradeForm
            appCurrentVersion={app.chart.metadata.version!}
            appCurrentValues={(app.config && app.config.raw) || ""}
            chartName={app.chart.metadata.name!}
            repo={repo}
            namespace={namespace}
            releaseName={releaseName}
            selected={selected}
            deployed={deployed}
            upgradeApp={upgradeApp}
            push={push}
            goBack={goBack}
            fetchChartVersions={fetchChartVersions}
            getChartVersion={getChartVersion}
          />
        </div>
      );
    }

    return (
      <SelectRepoForm
        isFetching={this.props.reposIsFetching}
        error={this.props.chartsError}
        kubeappsNamespace={this.props.kubeappsNamespace}
        namespace={this.props.namespace}
        repoError={this.props.repoError}
        repo={this.props.repo}
        repos={this.props.repos}
        chartName={app.chart?.metadata?.name!}
        checkChart={this.props.checkChart}
        fetchRepositories={this.props.fetchRepositories}
      />
    );
  }
}

export default AppUpgrade;
